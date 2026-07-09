package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/voocel/agentcore"
	"github.com/voocel/agentcore/llm"
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

// studioMaxTokens là trần output cho single-shot profile generation.
// Reasoning model cần đủ budget cho cả reasoning_content lẫn final content.
const studioMaxTokens = 16000

// Sync note: the long-novel survival rules below have UI companions in
// assets/app-production.js (profileLibCopyForLLM and buildProfileReviewPrompt).
// Keep the substance aligned; wording can differ because one generates and one reviews.
const profileStudioSystemPrompt = `You are a profile author for an autonomous long-form novel-writing engine.

Your job: turn the user's rough idea + parameters into ONE complete, production-ready
"profile" — the seed brief the engine's Architect expands into an outline and then hundreds of
chapters. This is commercial serialized fiction meant to be sold: be concrete and decisive, not
vague. Where the user left gaps, make strong creative choices and commit to them; the user edits
afterward.

Frame first (silently) — before writing anything, establish the frame below and let it drive every
choice. Do NOT print this analysis; the output is the profile only:
- Genre & sub-genre — exactly what the user asked for. HONOR it; never drift toward a default genre
  (e.g. romance / werewolf) when the user asked for something else.
- Saturation — is this genre/sub-genre already mass-market ("đại trà")? If yes, name the tired
  clichés you must avoid and where you will diverge; if niche, name what it must nail to satisfy its
  core fans.
- Genre conventions — the payoffs and beats readers of THIS genre expect and would feel cheated
  without; adapt everything below to them.
- Target market — the platform / market / reader given, plus the server-supplied current year;
  its tastes, trope codes and content limits.

Guiding principle — describe by PRINCIPLE, not by prescriptive example. State what each element
must ACHIEVE and the constraints it lives under, then let the downstream Architect and Writer
invent freely. Do NOT hand the Writer specimen sentences or ready-made prose to copy — that
anchors the model and kills creativity. The profile sets direction and boundaries, not wording.

Market & culture fit — this is commercial fiction aimed at a SPECIFIC market. Tailor genre
conventions, trope selection, romance heat / violence level, pacing, chapter length, and content
to the CURRENT tastes and reading culture of the target platform/market given by the user, at the
current year supplied in the request below. These differ sharply between markets (for instance
Spanish / Latin-American vs Vietnamese vs Anglo-American readers) in preferred tropes, 18+ /
violence thresholds, cultural-religious-legal taboos, and expected length & pace. Choose what
actually sells in that market now, and avoid tropes or content that are stale or culturally /
legally risky there. If no market is given, make broadly commercial choices for the story's
language. Do not name a year or cite trend statistics inside the profile itself; let market fit
shape the creative choices, not appear as meta-commentary.

Long-novel survival rules — genre-agnostic; these are what break at chapter 100+ if ignored. Apply
them whatever the genre (romance / cultivation / mystery / LitRPG / political fantasy / slice of
life / etc.), adapting only the vocabulary:
- Real costs, not just twists. A twist advances the PLOT; a cost hits the PROTAGONIST and sticks
  (a lost ally, broken trust, an irreversible sacrifice). Build in several genuine costs spread
  across the whole book, not one mid-point failure, and let each leave consequences that persist
  for many chapters. A protagonist who only ever wins stops being worried about.
- Bounded protagonist. Anchor where the protagonist's defining strength comes from, and name a few
  concrete blind spots it cannot cover. An unlimited strength kills tension.
- Named, active antagonists. Give at least one NAMED antagonist with their own scheme and the
  capacity to match the protagonist in direct, recurring confrontation — not only a faceless
  institution. Plant at least one betrayal or hidden thread and pay it off later.
- Serve the PRIMARY genre directly. Reserve dedicated beats for the lead genre's core payoff
  (relationship/longing for romance, the puzzle for mystery, progression for cultivation), so it
  is the main course, not a side effect of the plot engine. State the throughline that keeps that
  payoff alive across the whole book, including the emotional-satisfaction convention the target
  market expects for that genre.
- Name the moral tension. If the protagonist's methods contradict their stated ideals, name it as
  an ongoing crisis the story tracks, not a background trait.
- Lasting ensemble. Name and give a one-line arc to at least three supporting characters with
  their own stakes, so a long book does not run dry.
- Vary the rhythm. Do not lock every chapter into one template or one interior mode; allow quieter,
  non-strategic beats and occasional POV shifts. A single repeated interior mode is itself monotony.
- Concrete core mechanic. If the story has a central special mechanic (magic, bond, power, rule),
  define what it does, who controls it, and the cost/limit of using or breaking it; any prophecy or
  mystery planted early must have a defined payoff moment.
- Distinct voices. For each principal, give a voice PRINCIPLE — how they speak and react
  differently from the others — without writing specimen sentences, so a long book's dialogue
  never blurs.

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
  1. Title — ONE final title that pays off what the story delivers (title and ending must match).
  2. Genre & tone — primary genre + sub-genre, dominant emotional register, pace; target platform
     & reader if given.
  3. Reader promise — ONE sentence: the repeatable emotional or intellectual payoff every chapter
     must deliver. This is the engine's quality bar.
  4. Setting & world — the rules, costs and power limits that CONSTRAIN the plot, not decoration;
     enough for the Architect to derive world rules.
  5. Main characters — for each principal: want, wound, inner contradiction, and a distinct voice
     principle; for the protagonist also the SOURCE and the LIMITS/blind spots of their defining
     strength.
  6. Relationships — standing tension, not mere alliance; for the primary genre's central
     relationship, state its throughline (e.g. the longing that recurs even after the leads grow
     close).
  7. Story engine — the MECHANISM that pushes each chapter forward. State the repeating pattern:
     a solve-or-lose dilemma, an escalating threat, a mystery, a competition, a bond built through
     opposition — whichever fits.
  8. Core conflict & mid-pivot — the central pressure that sustains the WHOLE book, plus a single
     specific POINT-OF-NO-RETURN (roughly 40-60%) where the initial approach fails and strategy
     must fundamentally change.
  9. Costs & stakes — the concrete, lasting costs the protagonist pays across the book (distinct
     from twists); enough of them, spread across the arcs, to keep the outcome genuinely uncertain.
  10. Antagonists — at least one NAMED, active foil with their own scheme and recurring direct
     confrontation, plus a planted betrayal/thread and where it pays off.
  11. Ending direction (thematic) — the QUESTION the ending answers and the stance it takes (never
     a chapter name or plot beat); the irreversible cost paid to reach it (no free utopia); and how
     it pays off the title/premise.
  12. Chapter formula & rhythm — how a typical chapter opens, builds, and closes, PLUS the
     variations (quieter beats, POV shifts) that prevent monotony over hundreds of chapters.
  13. Differentiation hooks (>=3) — each something competing books in this market do NOT reliably
     deliver; flag any bold anti-trend choice as an intentional risk and how you compensate for it.
  14. What to avoid — genre clichés to skip, AND AI-tell stated as principles: no purple prose,
     no "not X but Y" construction, no repetitive interiority, no over-explaining dialogue, no
     uniform sentence rhythm.
  15. Length & scale — respect the requested chapter count; sketch how arc load and the costs above
     distribute across the run.
- Keep it tight and usable — a strong brief, not a draft of the novel. ~700-1100 words.
- Prefer stating intentions and boundaries over writing specimen prose.
- Before final output, silently rewrite the profile to (a) remove every banned phrase, placeholder
  alternative, and "not X but Y" / "không phải X mà là Y" construction — even inside worldbuilding —
  and (b) confirm each long-novel survival rule above is actually reflected.`

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
		Model          string `json:"model"`
		Provider       string `json:"provider"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(body.Idea) == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("idea is required"))
		return
	}

	providerOverride := strings.TrimSpace(body.Provider)
	modelOverride := strings.TrimSpace(body.Model)
	if (providerOverride == "") != (modelOverride == "") {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("provider and model must be provided together"))
		return
	}

	// If user specified a different provider/model, create a one-off model for this request
	var model agentcore.ChatModel
	if providerOverride != "" {
		pc, ok := s.cfg.Providers[providerOverride]
		if !ok {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("unknown provider: %s", providerOverride))
			return
		}
		providerType, err := pc.ProviderType(providerOverride)
		if err != nil {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("provider type: %w", err))
			return
		}
		providerExtra := cloneMap(pc.Extra)
		if pc.API != "" {
			if providerExtra == nil {
				providerExtra = make(map[string]any, 1)
			}
			providerExtra["api"] = pc.API
		}
		m, err := llm.NewModel(providerType, modelOverride,
			llm.WithAPIKey(pc.APIKey),
			llm.WithBaseURL(pc.BaseURL),
			llm.WithStreamIdleTimeout(5*time.Minute),
			llm.WithProviderExtra(providerExtra),
			llm.WithExtra(pc.ExtraBody),
		)
		if err != nil {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("model init: %w", err))
			return
		}
		model = m
	} else {
		ms, err := s.studioModelSet()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, fmt.Errorf("model init: %w", err))
			return
		}
		model = ms.ForRole("thinking")
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
	// Anchor "current tastes" to the real current year so the model fits the
	// target market as of now, not its training-cutoff sense of "current".
	fmt.Fprintf(&b, "Current year (fit market tastes to this moment): %d\n", time.Now().Year())

	// Stream the profile back to the browser as SSE so data keeps flowing through
	// the LTN proxy (3-min timeout) instead of stalling until the full brief is
	// ready. A non-streaming response gets 504'd when generation exceeds the
	// proxy timeout; SSE deltas arrive continuously and reset that clock.
	flusher, ok := w.(http.Flusher)
	if !ok {
		// No streaming support (e.g. httptest without flushing): fall back to the
		// old single-shot JSON so the request still completes.
		content, err := s.runProfileGeneration(r.Context(), model, b.String())
		if err != nil {
			classified := agentcore.ClassifyProvider(err)
			if errors.Is(classified, agentcore.ErrProviderTimeout) || errors.Is(classified, agentcore.ErrProviderStreamIdle) {
				writeErr(w, http.StatusGatewayTimeout, fmt.Errorf("profile generate timed out; try a faster Studio model or a shorter brief"))
				return
			}
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"content": content})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	streamCh, err := s.runProfileStream(r.Context(), model, b.String())
	if err != nil {
		classified := agentcore.ClassifyProvider(err)
		if errors.Is(classified, agentcore.ErrProviderTimeout) || errors.Is(classified, agentcore.ErrProviderStreamIdle) {
			writeSSE(w, flusher, sseMessage{Type: "profileError", Text: "profile generate timed out; try a faster Studio model or a shorter brief"})
		} else {
			writeSSE(w, flusher, sseMessage{Type: "profileError", Text: err.Error()})
		}
		flusher.Flush()
		return
	}

	var out strings.Builder
	var thinking strings.Builder
	var streamed bool
	for ev := range streamCh {
		switch ev.Type {
		case agentcore.StreamEventThinkingDelta:
			// Reasoning models (glm-5.2 etc.) emit a long reasoning_content phase
			// before the final answer. Forward a heartbeat so the browser knows the
			// model is still working (not stalled) — but do NOT pour the raw
			// reasoning into the profile textarea; only the final content goes there.
			thinking.WriteString(ev.Delta)
			if !writeSSE(w, flusher, sseMessage{Type: "profileThinking"}) {
				return // client disconnected
			}
		case agentcore.StreamEventTextDelta:
			streamed = true
			out.WriteString(ev.Delta)
			if !writeSSE(w, flusher, sseMessage{Type: "profileDelta", Text: ev.Delta}) {
				return // client disconnected
			}
		case agentcore.StreamEventDone:
			if !streamed {
				out.WriteString(ev.Message.TextContent())
			}
		case agentcore.StreamEventError:
			if ev.Err != nil {
				classified := agentcore.ClassifyProvider(ev.Err)
				if errors.Is(classified, agentcore.ErrProviderTimeout) || errors.Is(classified, agentcore.ErrProviderStreamIdle) {
					writeSSE(w, flusher, sseMessage{Type: "profileError", Text: "profile generate timed out; try a faster Studio model or a shorter brief"})
				} else {
					writeSSE(w, flusher, sseMessage{Type: "profileError", Text: ev.Err.Error()})
				}
				flusher.Flush()
				return
			}
			writeSSE(w, flusher, sseMessage{Type: "profileError", Text: "profile generate failed"})
			flusher.Flush()
			return
		}
	}

	text := strings.TrimSpace(out.String())
	// Reasoning-model fallback: a model can write its whole answer into
	// reasoning_content and never switch to the final-answer channel (see the
	// same guard in internal/host/cocreate.go). Salvage thinking as the profile
	// so the user still gets output instead of an empty-profile error.
	if text == "" {
		if t := strings.TrimSpace(thinking.String()); t != "" {
			text = t
		}
	}
	text = stripCodeFence(text)
	if text == "" {
		writeSSE(w, flusher, sseMessage{Type: "profileError", Text: "model returned empty profile"})
		flusher.Flush()
		return
	}
	writeSSE(w, flusher, sseMessage{Type: "profileDone", Text: text})
	flusher.Flush()
}

// runProfileStream opens the streaming LLM call and returns the raw stream
// channel. It sets NO fixed deadline: callers bound it via the passed context
// (the SSE handler uses r.Context(), so a browser abort cancels the stream)
// plus the model's stream-idle watchdog. This lets SSE deltas flow without a
// hard Go deadline cutting a long generation short.
func (s *server) runProfileStream(ctx context.Context, model agentcore.ChatModel, userMsg string) (<-chan agentcore.StreamEvent, error) {
	msgs := []agentcore.Message{
		agentcore.SystemMsg(profileStudioSystemPrompt),
		agentcore.UserMsg(userMsg),
	}
	streamCh, err := model.GenerateStream(ctx, msgs, nil, agentcore.WithMaxTokens(studioMaxTokens))
	if err != nil {
		return nil, fmt.Errorf("profile generate: %w", err)
	}
	return streamCh, nil
}

// runProfileGeneration does one LLM call (drains the stream server-side) and
// returns the trimmed markdown. Used by the non-SSE fallback path; the SSE path
// drains runProfileStream directly to forward deltas to the browser.
func (s *server) runProfileGeneration(ctx context.Context, model agentcore.ChatModel, userMsg string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()
	streamCh, err := s.runProfileStream(ctx, model, userMsg)
	if err != nil {
		return "", err
	}

	var out strings.Builder
	var thinking strings.Builder
	var streamed bool
	for ev := range streamCh {
		switch ev.Type {
		case agentcore.StreamEventThinkingDelta:
			thinking.WriteString(ev.Delta)
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
	if text == "" {
		if t := strings.TrimSpace(thinking.String()); t != "" {
			text = t
		}
	}
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

// cloneMap returns a shallow copy of m. Used to avoid mutating shared
// ProviderConfig.Extra when building a one-off model override.
func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
