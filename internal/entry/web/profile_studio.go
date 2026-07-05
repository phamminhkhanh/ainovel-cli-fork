package web

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/voocel/agentcore"
	"github.com/voocel/ainovel-cli/internal/bootstrap"
)

// Profile Studio (C-lite): turn a rough idea + a few fields into a complete,
// production-ready profile.md in ONE LLM call, using a system prompt tuned for
// commercial serialized fiction — the things co-create's generic brief prompt
// does not ask about (platform, monetization angle, AI-tell avoidance).
//
// Additive: builds its own bootstrap.ModelSet from the web adapter's cfg
// (bootstrap.NewModelSet is exported); it does NOT touch internal/host. The
// output is NOT auto-used — it lands in the editable Profile Library textarea,
// so the user reviews/edits/saves it as an SSOT artifact before any run exists.
//
// This is single-shot on purpose. Multi-turn interview + strict JSON schema is
// deferred (C-full) until this proves valuable.

// studioModelSet lazily builds and caches a ModelSet from the web STARTUP cfg.
// Note: this is independent of the Host's own model set, so a runtime model
// switch via /api/model does NOT affect Studio — it uses the config the server
// booted with. (A per-Studio model dropdown could be added later if needed.)
// Cached error is fine: NewModelSet failure is config-deterministic, not transient.
func (s *server) studioModelSet() (*bootstrap.ModelSet, error) {
	s.studioOnce.Do(func() {
		s.studioModels, s.studioErr = bootstrap.NewModelSet(s.cfg)
	})
	if s.studioErr != nil {
		return nil, s.studioErr
	}
	return s.studioModels, nil
}

const profileStudioSystemPrompt = `You are a profile author for an autonomous long-form novel-writing engine.

Your job: turn the user's rough idea + parameters into ONE complete, production-ready
"profile" — the seed brief the engine's Architect expands into an outline and then hundreds of
chapters. This is commercial serialized fiction meant to be sold: be concrete and decisive, not
vague. Where the user left gaps, make strong creative choices and commit to them; the user edits
afterward.

Guiding principle — describe by PRINCIPLE, not by prescriptive example. State what each element
must ACHIEVE and the constraints it lives under, then let the downstream Architect and Writer
invent freely. Do NOT hand the Writer specimen sentences or ready-made prose to copy — that
anchors the model and kills creativity. The profile sets direction and boundaries, not wording.

Hard rules (violating these makes the profile worse, not richer):
- COMMIT to one concrete choice everywhere. Never offer alternatives like "A or B", "A hoặc B",
  or "default A/B" and never defer a decision back to the user. Decide, and write the decision.
- Do NOT include the word "ví dụ" / "e.g." / "for example", quoted sample sentences, or any
  specimen prose anywhere. Describe only what each element must achieve.
- The profile's OWN wording must obey its own "what to avoid" list. In particular, never use the
  "không phải X mà là Y" / "not X but Y" construction anywhere in the profile itself.

Output rules:
- Output ONLY the profile as Markdown. No preamble, no explanation, no code fences.
- Write in the user's requested language (default: Vietnamese). Keep genre / trope names natural.
- Use "## " headings. Cover, adapted to the story:
  1. Title — 2-3 candidate titles taking different angles.
  2. Genre & tone — primary genre + sub-genre, dominant emotional register, pace; target
     platform & reader if given.
  3. Setting & world — the rules and texture that CONSTRAIN the plot (resources, costs, power
     boundaries), not decoration; enough for the Architect to derive world rules.
  4. Main characters & relationships — for each principal: a want, a wound, and an inner
     contradiction; relationships must carry standing tension, not mere alliance.
  5. Core conflict — the central pressure that can sustain the WHOLE book, not one episode.
  6. Ending direction (thematic) — the QUESTION the ending answers and the stance it takes;
     never a chapter name or plot beat.
  7. Chapter formula — how a typical chapter opens, builds, and closes; the discipline that
     keeps readers turning the page.
  8. Differentiation hooks (>=3) — each must be something competing books in this genre do NOT
     reliably deliver.
  9. What to avoid — genre clichés to skip, AND AI-tell stated as principles: no purple prose,
     no "not X but Y" construction, no repetitive interiority, no over-explaining dialogue, no
     uniform sentence rhythm.
  10. Length & scale — respect the requested chapter count; sketch how arc load distributes
     across the run.
- Keep it tight and usable — a strong brief, not a draft of the novel. ~600-1000 words.
- Prefer stating intentions and boundaries over writing specimen prose.`

// handleProfileGenerate runs the single-shot generation and returns markdown.
// The result is returned only — the client puts it in the Library editor for
// review; nothing is saved or run automatically.
func (s *server) handleProfileGenerate(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	var body struct {
		Idea           string `json:"idea"`
		Language       string `json:"language"`
		Genre          string `json:"genre"`
		Platform       string `json:"platform"`
		StyleNotes     string `json:"styleNotes"`
		TargetChapters int    `json:"targetChapters"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(body.Idea) == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("idea is required"))
		return
	}

	ms, err := s.studioModelSet()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, fmt.Errorf("model init: %w", err))
		return
	}

	lang := strings.TrimSpace(body.Language)
	if lang == "" {
		lang = "Vietnamese"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Rough idea:\n%s\n\n", strings.TrimSpace(body.Idea))
	fmt.Fprintf(&b, "Output language: %s\n", lang)
	if g := strings.TrimSpace(body.Genre); g != "" {
		fmt.Fprintf(&b, "Genre: %s\n", g)
	}
	if p := strings.TrimSpace(body.Platform); p != "" {
		fmt.Fprintf(&b, "Target platform / market: %s\n", p)
	}
	if body.TargetChapters > 0 {
		fmt.Fprintf(&b, "Target length: about %d chapters\n", body.TargetChapters)
	}
	if sn := strings.TrimSpace(body.StyleNotes); sn != "" {
		fmt.Fprintf(&b, "Style notes / must-haves: %s\n", sn)
	}

	content, err := s.runProfileGeneration(r.Context(), ms, b.String())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"content": content})
}

// runProfileGeneration does one non-streaming LLM call (drains the stream
// server-side) and returns the trimmed markdown.
func (s *server) runProfileGeneration(ctx context.Context, ms *bootstrap.ModelSet, userMsg string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 150*time.Second)
	defer cancel()

	model := ms.ForRole("thinking")
	msgs := []agentcore.Message{
		agentcore.SystemMsg(profileStudioSystemPrompt),
		agentcore.UserMsg(userMsg),
	}
	streamCh, err := model.GenerateStream(ctx, msgs, nil, agentcore.WithMaxTokens(4096))
	if err != nil {
		return "", fmt.Errorf("profile generate: %w", err)
	}

	var out strings.Builder
	var streamed bool
	for ev := range streamCh {
		switch ev.Type {
		case agentcore.StreamEventTextDelta:
			streamed = true
			out.WriteString(ev.Delta)
		case agentcore.StreamEventDone:
			if !streamed {
				out.WriteString(ev.Message.TextContent())
			}
		case agentcore.StreamEventError:
			if ev.Err != nil {
				return "", fmt.Errorf("profile generate: %w", ev.Err)
			}
			return "", fmt.Errorf("profile generate failed")
		}
	}

	text := strings.TrimSpace(out.String())
	// Strip an accidental ```markdown fence if the model wrapped the output.
	text = stripCodeFence(text)
	if text == "" {
		return "", fmt.Errorf("model returned empty profile")
	}
	return text, nil
}

// stripCodeFence removes a single wrapping ```lang ... ``` fence if present.
func stripCodeFence(s string) string {
	if !strings.HasPrefix(s, "```") {
		return s
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[i+1:]
	}
	s = strings.TrimSuffix(strings.TrimRight(s, "\n"), "```")
	return strings.TrimSpace(s)
}
