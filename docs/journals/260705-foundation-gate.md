# Journal — Foundation Gate (Production Cockpit)

**Date:** 2026-07-05
**Feature:** Foundation Gate — pause a fresh production run after the foundation is planned so the
user can review / hand-edit / regenerate it before the bulk of the book is written.
**Design doc:** [`docs/de-xuat-cai-tien-chat-luong.md`](../de-xuat-cai-tien-chat-luong.md) §1
**Note on naming:** the design doc splits this into "Milestone 1a" (review/approve/reject) and
"1b" (steer & retry). In practice this journal covers 1a **plus** the 1b regenerate-with-feedback
path (Revise) and a manual-edit path (Reveal). The only 1b variant deliberately NOT built is
steering into a live Host (would break additive-only). The "milestone" numbers are planning
labels; this journal is the shipped record.

## What changed

Production Cockpit now auto-pauses a `fresh_profile` run after the Architect finishes the
foundation (premise/outline/world/characters), so the user can review it before the bulk of the
book is written. At the pause the user can **Approve** (resume), **Reject** (discard),
**hand-edit** the foundation files then approve (via a Reveal-folder button), or **Revise**
(regenerate the foundation from the profile + a steering note).

> **This is a best-effort pause, not a hard gate.** Detection is a 5s poll of `progress.json`.
> The engine flips `phase` to `writing` *synchronously* inside `save_foundation`, and the flow
> dispatcher immediately `Steer`s a "write chapter 1" instruction. So in the worst case the Writer
> may already have started (or partially drafted) chapter 1 before the next poll tick lands. In
> practice a full chapter takes many model calls / tens of seconds, so the pause almost always
> lands during chapter 1's drafting — worst-case waste is a partial single chapter, never the
> hundreds of chapters the gate exists to protect. A truly deterministic zero-token gate would
> require a stop hook inside `internal/host`/`headless` (breaking additive-only); deliberately not
> done — see design doc §1 and the code-review verdict.

"Revise" is the additive form of steer-and-retry: it does NOT steer a live Coordinator (there is
none — the child has exited). It creates a NEW fresh_profile run from the same profile with the
note appended to the prompt, regenerating the whole foundation. It is a re-roll with guidance, not
a surgical edit — the UI says so explicitly. For surgical edits, use the hand-edit (Reveal) path.

### Backend

- `internal/entry/web/prodrun.go` — new `prodRunAwaitingReview` status and `stopReasonFoundationReady`.
- `internal/entry/web/prodrun_runner.go`:
  - `poll()` detects the foundation→writing transition (`phase=="writing"` with 0 completed
    chapters, `fresh_profile` kind only, not yet approved) and kills the child into
    `awaiting_review`. Best-effort (5s poll) — see the pause caveat above.
  - `readWorkspacePhase()` — reads `Progress.Phase` from a run's `progress.json`.
  - `runDirHasExistingOutput()` — lets `start()` skip `--prompt-file` on approve-resume (the run
    dir already has a seeded book at phase=writing; headless must go through native `Resume()`).
  - `prodRunManager.ApproveFoundation()` — re-queues and restarts the same run dir.
  - `prodRunManager.ReviseFoundation()` — creates a new run from the same profile with the
    steering note (`ProdRun.RevisionNote`) and starts it; the old awaiting_review run is **kept**
    as a fallback (see round-3 fix below).
  - `startWithReapRetry()` — shared retry helper used by both approve and revise for the
    kill-reap window; matches the typed `errAnotherRunActive` sentinel, not a message substring.
- `internal/entry/web/prodrun_handlers.go` — `GET /api/prodruns/{id}/foundation` (reuses
  `serveOutline`/`serveWorld`/`serveCharacters` from `content.go` against the run's own sandboxed
  `output/novel` via a small `runStoreEngine` adapter), and `POST .../{approve,reject,revise,reveal}`.
  `revise` appends the note to `profile.md`; `reveal` opens `runDir/output/novel` via `revealOpen`
  (loopback-only, same guard as `handleReveal`).
- `internal/entry/web/server.go` — 5 new routes registered (foundation, approve, reject, revise, reveal).

### Frontend

- `internal/entry/web/assets/app-production.js` — `awaiting_review` status label, review notice
  banner (honest best-effort wording), Approve/Reject buttons; foundation preview showing premise +
  **layered outline (Volume→Arc themes, where the twists live)** + full chapter list + character
  list + world-rule list; a **manual-edit block** (guidance + "Mở thư mục nền móng" Reveal button)
  and an **AI-revise block** (warning that it rewrites the whole foundation + note textarea).
  Preview is cached per run id so the 5s poll no longer flickers "Đang tải…" or re-fetches.
- `internal/entry/web/assets/app.css` — `.run-badge-awaiting_review`, `.run-review-notice`,
  `.run-foundation-preview`, `.foundation-section`/`.foundation-list`/`.foundation-vol`/`-theme`/
  `-final`, `.run-revise-box`/`.run-revise-warn`, `.run-manual-box`, `.run-edit-title`.

## Key decisions

- **Detection is poll-based, not event-based.** There is no `foundation_ready` event — phase
  transitions live only in `save_foundation`'s tool result + `progress.json`. Cockpit already
  polls `progress.json` every 5s for chapter/review/cost stats; reusing that loop avoids inventing
  a new signal path.
- **`continue_workspace` runs are excluded from the gate.** They're seeded already at
  `phase=writing`, so the same check would immediately mis-fire on start.
- **Approve = restart the same run dir, not a new continue_workspace job.** The run's own
  `output/novel` already has the seeded foundation at `phase=writing`; `runDirHasExistingOutput`
  makes the runner skip `--prompt-file` so headless enters native `Resume()` instead of
  `startup.PrepareQuick` on the same profile.
- **Reject just deletes** (reuses `prodRunManager.Delete`), gated to `awaiting_review` only.

## Fixed during adversarial code review

- **Critical: infinite re-gate loop.** `poll()` detects "foundation just saved" purely from
  `progress.json` state (`phase=="writing"`, 0 chapters). After `ApproveFoundation` restarts the
  same run dir, that file stays in exactly that state for as long as the real Writer takes to
  draft+commit chapter 1 — routinely longer than one 5s poll tick. Without memory of the approval,
  the very next poll would kill the freshly-approved run again, forever, before it ever got to
  write a single chapter. **Proved with a throwaway probe test** (approve, then poll while the
  child was still alive and progress.json unchanged) before fixing — confirmed `awaiting_review`
  came back within one tick. **Fix:** added `ProdRun.FoundationApproved bool`; `poll()`'s gate
  check now skips runs where it's true; `ApproveFoundation` sets it. Added a permanent regression
  test, `TestRunnerPollDoesNotReGateAfterApprove`.
- **Stale comments.** `prodRunAwaitingReview`'s doc comment and `handleProdRunApprove`'s doc
  comment both said "Approve re-queues a continue_workspace run" — the actual implementation
  restarts the *same* run dir with kind still `fresh_profile`. Comments corrected to match code.

## Fixed in code review round 2

- **Dishonest "zero Writer token" claim.** UI copy, journal, and code comments promised the pause
  stops before any Writer token. That's not guaranteed (see best-effort caveat above). Reworded
  everywhere to "pauses before the bulk of the book; worst case a partial chapter 1." Decision:
  keep web-only best-effort rather than add a host-side stop hook — the fork's whole value is
  near-zero upstream-merge conflict, not worth trading for a ~1-chapter savings.
- **Foundation preview too thin to actually review.** It only fetched outline + world and showed
  counts, while the UI told the user to review characters too. Now fetches all three sections
  (`?section=characters` was already served by the backend) and renders the full outline list,
  character list, and world-rule list.
- **Approve race with the kill-reap window.** `poll()`'s `killProcess` is async; `waitProc`
  removes the run from `rr.running` only after `cmd.Wait()`. Approving within that window made
  `start()` fail with "already running." `ApproveFoundation` now retries `start()` for ~1s through
  that window. Regression test: `TestApproveFoundationRetriesThroughReapWindow`.
- **Approve failed if the source profile was moved/deleted** (found by a live smoke test, missed
  by unit tests that always seeded a valid profile). `prepareRunDir`'s `fresh_profile` branch
  re-resolved and re-copied `profile.md` on *every* start, including approve-resume — but
  approve-resume Resume()s from the seeded run dir and never uses the profile. A missing profile
  made approve fail with `status=failed` instead of resuming. Fix: skip the profile copy when
  `runDirHasExistingOutput(runDir)` is true. Regression test:
  `TestApproveFoundationResumesEvenIfProfileDeleted`.

## Fixed in code review round 3 (Revise / dual QA)

- **Blocker: Revise deleted the reviewed candidate too early.** The first cut created the new run,
  started it, then immediately deleted the old `awaiting_review` run. But the new run only reaches
  `awaiting_review` after regenerating its foundation (~tens of seconds, can fail on API/budget/
  profile). Deleting the old candidate before that = data loss of a good foundation on failure.
  **Fix:** `ReviseFoundation` now KEEPS the old run; the user picks the new one and rejects the old
  when satisfied. On start failure the dead new run is cleaned up, old stays intact.
- **Fragile error-string matching → typed sentinel.** `start()` now returns `errAnotherRunActive`;
  the retry helper matches with `errors.Is`, not `strings.Contains(..., "already running")`.
- **Revise lacked the reap-window retry** that approve had. Both now share `startWithReapRetry`.
- **Approve failure stranded the run in `queued`** (could no longer be rejected). `ApproveFoundation`
  now reverts to `awaiting_review` on start failure.
- **Residual "no Writer tokens" overclaims** in code comments, UI copy, `01-TONG-QUAN §9.4`, and the
  260703 state-machine/table. Reworded: revise/gate are best-effort, worst case a partial chapter 1;
  "reject" genuinely spends 0 Writer tokens (never wrote) and keeps that wording.

## Live smoke test (real binary, real HTTP, no LLM spend)

Built the binary, booted `--web` against a hand-seeded `awaiting_review` run, and exercised the
new endpoints over real HTTP (not httptest):

- `GET /api/prodruns` — the seeded `awaiting_review` run survived `load()` across startup (not
  coalesced to failed like running/paused), confirming the deliberate exclusion from the
  unclean-shutdown rule.
- `GET /api/prodruns/{id}/foundation` (+ `?section=world`, `?section=characters`) — returned real
  premise/outline/characters/world with Vietnamese UTF-8 intact (no mojibake) over the wire.
- `POST /api/prodruns/{id}/reject` — run removed from the list AND its run dir deleted from disk
  on Windows (validates the reject→Delete→RemoveAll path the QA report flagged).
- Nonexistent-run and wrong-method requests returned the expected 404s.
- Approve happy-path was NOT run live (it spawns a real headless child → real LLM tokens); it is
  covered by unit tests. The live run is what surfaced the missing-profile approve bug above.

## Verification

- `go build ./...` — pass
- `go vet ./...` — pass
- `go test ./internal/entry/web/...` — pass, including new tests:
  `TestRunnerPollDetectsFoundationReady`, `TestRunnerPollIgnoresFoundationReadyForContinueWorkspace`,
  `TestApproveFoundationResumesRunDir`, `TestApproveFoundationRejectsWrongStatus`,
  `TestHandleProdRunFoundationServesOutline`, `TestHandleProdRunFoundationMissingRun`,
  `TestHandleProdRunReject`, `TestHandleProdRunRejectRejectsWrongStatus`,
  `TestHandleProdRunApproveWrongStatus`, plus revise/reveal:
  `TestReviseFoundationCreatesNewRunWithNote`, `TestReviseFoundationRejectsEmptyOrWrongStatus`,
  `TestHandleProdRunReviseValidation`, `TestHandleProdRunRevealOpensFoundationDir`,
  `TestHandleProdRunRevealBlockedOnPublicBind`, and the re-gate/reap/profile regressions
  (`TestRunnerPollDoesNotReGateAfterApprove`, `TestApproveFoundationRetriesThroughReapWindow`,
  `TestApproveFoundationResumesEvenIfProfileDeleted`)
- `node --check` on `app-production.js` — pass
- Repeated `-count=10` on the approve-resume test to rule out the Windows log-file-lock race seen
  during initial iteration (fixed by waiting on `onCmdFinished` before asserting, matching the
  pattern already used by `TestRunnerStopKillsProcess`).

## Caveats / left for later

- **Revise is a re-roll, not a surgical edit.** It regenerates the whole foundation from
  profile + note; names/chapters/other volumes may all change. For precise changes use the
  hand-edit (Reveal) path. Steering into a *live* Host (keep the engine paused and Steer) is the
  one 1b variant deliberately NOT built — it would break additive-only.
- The gate only fires for `fresh_profile`. Reviewing a new `expand_arc`/`append_volume` mid-book
  is a different detection problem (deferred per design doc §1).
- Hand-edit is powerful but unguarded: editing chapter *counts* can desync
  `outline.json`↔`layered_outline.json`↔`progress.json`. The UI warns; there is no validation.
- No `theme`/`lesson` field exists in `save_foundation`; the preview surfaces premise + layered
  outline themes + chapters + characters + world rules, but no dedicated theme/lesson summary.
- `FoundationApproved` is one-way and per-run: if a run is somehow re-queued from `queued` without
  going through `ApproveFoundation` (not currently possible via the API, but worth flagging for
  future refactors), the gate would fire again as a fresh run — which is actually the *correct*
  fallback behavior, not a bug.
