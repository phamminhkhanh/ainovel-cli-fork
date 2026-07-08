# Journal — Profile Library & Profile Studio (Production Cockpit)

**Date:** 2026-07-05
**Feature:** Author, review, store and AI-generate production **profiles** entirely in the Web UI,
so the user never hand-edits files outside the app.
**Design doc:** [`docs/de-xuat-cai-tien-chat-luong.md`](../de-xuat-cai-tien-chat-luong.md)
**Related:** [`260705-foundation-gate.md`](260705-foundation-gate.md) (a run created from a profile
pauses at the Foundation Gate for review).

## Why

A profile is the **SSOT seed brief** a production run is created from. Previously the only way to
get one was to hand-write a `.md` file into `./.ainovel/profiles/` outside the app. This adds two
in-UI authoring paths, keeping the principle: **the profile is authored/reviewed/stored BEFORE a
run exists — never auto-generated implicitly inside job creation.**

## What changed

### Profile Library (manage .md profiles in the UI)

- `internal/entry/web/profiles_library.go` (new):
  - `GET /api/profiles/content?path=…` — read one profile (validated + within-dir + symlink-guarded).
  - `POST /api/profiles/save` — create/edit. Writes ONLY to the project dir (`./.ainovel/profiles/`).
    Name is sanitized to a bare `.md` filename (rejects path separators / traversal / dot-only).
    **Never silently clobbers**: if the file exists and `overwrite` is not set → `409`.
  - `POST /api/profiles/delete` — **project-only**. Enforced by checking the RESOLVED path is inside
    the project dir (`isWithinDir`), so a crafted `global/…` or `legacy/…` ref returns `403` and
    cannot delete the user's global profiles or the repo's samples.
- UI (`app-production.js`): a "📚 Thư viện Profile" modal — list (project editable, global/legacy
  read-only), view/edit in a textarea, Save, Delete, "+ Profile mới". New prefills a
  **principle-based template** (9 sections, abstract hints, no concrete story) + a "📖 Hướng dẫn
  & lưu ý" note kept OUT of the saved file.

### Profile Studio (C-lite — generate a profile from a rough idea)

- `internal/entry/web/profile_studio.go` (new):
  - `POST /api/profiles/generate` — one-shot: build a `bootstrap.ModelSet` from the web adapter's
    `cfg` (`bootstrap.NewModelSet`, exported — no host coupling), call `ForRole("thinking")` with a
    custom system prompt tuned for commercial serialized fiction, return the markdown.
  - Output is returned only — it lands in the Library editor for review; nothing is saved or run.
- UI: a "✨ Sinh profile từ ý tưởng" block (idea + genre/platform/language/chapters/style) → fills
  the editor for the user to review/edit/save.

## Key decisions

- **Profile = SSOT, authored before the run.** Studio never auto-saves or auto-runs; its output is
  a draft in the editable Library, reviewed like any artifact.
- **Additive.** All new code is in `internal/entry/web/`; only route registration touched in
  `server.go` (already a fork-exception file). `bootstrap.NewModelSet(cfg)` is a public constructor —
  calling it is consuming an exported API, not modifying upstream. No `internal/host` change.
- **Describe by PRINCIPLE, not example.** Both the Studio system prompt and the manual template
  state what each element must ACHIEVE and its constraints, and deliberately avoid concrete story
  examples — LLMs anchor on examples, which kills downstream creativity. (User-driven principle.)
- **C-lite, not C-full.** Single-shot generation + markdown output (not multi-turn interview, not
  JSON-mode/schema). Deferred until this proves valuable.
- **Reused, not reinvented.** `serveOutline`/`serveWorld`/`serveCharacters` and the profile-path
  resolver / `sanitizeFileName` are reused; the Studio model set mirrors the engine's own
  `NewModelSet(cfg)` so there is no model-config drift.

## Fixed in code review (dual QA)

- **Critical: delete allowed global/legacy.** `handleProfileDelete` resolved any source then removed
  it — a crafted `POST /api/profiles/delete {"path":"global/foo.md"}` could delete
  `~/.ainovel/profiles/foo.md`, contradicting the project-only contract. Fixed with an
  `isWithinDir(resolved, projectBase)` guard → `403` for non-project. Tests:
  `TestHandleProfileDeleteRejectsNonProject` (global/legacy/old-legacy).
- **Silent overwrite.** Save now returns `409` when the file exists without `overwrite:true`; the UI
  confirms then retries. Test: `TestHandleProfileSaveNoSilentOverwrite`.
- **Dot-only names.** `profileFileName(".")` would create a hidden `.md`; now rejected (400).

## Verification

- `go build ./...` / `go vet ./...` / `gofmt -l` — pass / clean
- `node --check app-production.js` — pass
- `go test ./internal/entry/web/...` — pass, incl. new tests:
  `TestHandleProfileSaveThenContent`, `TestHandleProfileSaveNoSilentOverwrite`,
  `TestHandleProfileSaveValidation` (incl. dot-only), `TestHandleProfileContentRejectsUnsafePath`,
  `TestHandleProfileDelete`, `TestHandleProfileDeleteMissing`,
  `TestHandleProfileDeleteRejectsNonProject`, `TestHandleProfileGenerateRequiresIdea`,
  `TestStripCodeFence`.

## Caveats / left for later

- The successful `generate` path is NOT unit-tested (needs a configured provider / real model call;
  hard to mock). Validation + `stripCodeFence` are tested; the happy path is exercised live.
- Studio uses the **startup config** model set (`bootstrap.NewModelSet(cfg)`), independent of the
  Host's runtime `/api/model` switch — switching the engine's model at runtime does not change
  which model Studio generates with. A per-Studio model dropdown could be added later.
- Prompt-adherence hardened after review: the system prompt now forbids "A or B" alternatives,
  forbids "ví dụ"/specimen prose, and requires the profile to obey its own avoid-list (no
  "không phải X mà là Y"). Round-2 live test scorecard: the "commit / no A-or-B" and
  "no specimen prose" rules now hold; **the self-adherence rule still slips** — the model keeps
  reaching for the "không phải X mà là Y" contrastive frame in thematic lines (the Ending
  direction), even while its own avoid-list forbids it.
  **Known limitation (accepted, not fixed):** negative instructions about one idiomatic linguistic
  pattern have diminishing returns; hardening the prompt further isn't worth the churn. Low harm:
  the slip lives in the profile's *thematic description* (read by the Architect), not in prose the
  Writer copies, and the avoid-list still correctly instructs the Writer against the pattern. The
  **review-before-save step is the real gate** — the user edits such lines out in seconds. Decision:
  leave as best-effort; do NOT add server-side rewriting of model output.
- C-full (multi-turn interview + JSON-mode + strict field validation) is deferred.
- Studio always generates into the **project** dir on save; global/legacy remain read-only (edit =
  save-as into project).
- Step 4 review checklist remains a **UI/human gate**, not server-side validation. This is
  intentional: profile quality is semantic and market-specific; a hard server validator would become
  a brittle policy engine. The safer contract is generate/copy/review → editable textarea → explicit
  save.
