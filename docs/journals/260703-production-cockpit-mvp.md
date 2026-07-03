# Journal — Production Cockpit MVP

**Date:** 2026-07-03  
**Feature:** Production Cockpit MVP ("Sản xuất" tab)  
**Plan:** `plans/260703-1338-production-cockpit-mvp/plan.md`

## What changed

Implemented a new Web UI tab that lets users queue and run automated, headless novel-generation jobs as separate child `ainovel-cli --headless` processes.

### Backend

- `internal/entry/web/prodrun.go` — `ProdRun` model and JSON store with unclean-shutdown recovery.
- `internal/entry/web/prodrun_runner.go` — child-process spawn, filesystem polling, hard target-chapters stop, pause detection, orphan PID tracking.
- `internal/entry/web/prodrun_handlers.go` — `/api/profiles`, `/api/prodruns*` endpoints.
- `internal/entry/web/prodrun_export.go` — server-side TXT concatenation of chapter `.md` files.
- Wired into `server.go` and `run.go` (fork-exception files already modified by this fork).

### Frontend

- `internal/entry/web/assets/app-production.js` — tab logic.
- Updated `index.html`, `app-workspace.js`, `app-i18n.js`, `app.css`.
- Added `app-production.js` to `embed.go` and `assets_test.go` guards.

## Key decisions

- **Child-process isolation.** The Web UI already owns a `host.Host`, so each run spawns its own `ainovel-cli --headless` process with `Cmd.Dir` set to the run directory. This forces engine output into `{runDir}/output/novel` because `bootstrap.Config.OutputDir` is `json:"-"` and `FillDefaults()` resolves it relative to cwd.
- **Hard target-chapters stop.** The engine has no `max_chapters` config. The runner polls `meta/progress.json` and kills the child when `len(completed_chapters) >= targetChapters`.
- **Read-only pause.** Pause markers in `run.log` change status to `paused`; MVP offers only Stop/Export, no Continue.
- **TXT export only.** Export concatenates `chapters/*.md` server-side; it does not call `s.eng.Export()`, which exports the Web UI's own novel store.
- **Orphan awareness.** `ChildPID` is saved; on restart any previously `running` run becomes `failed`/`unclean_shutdown` with `PossiblyOrphaned: true`.

## QA fixes during review

| Finding | Fix |
|---------|-----|
| `app-i18n.js` syntax error (`translateRoleTags` broken by inserted `PROD_LABELS`) | Restored function; removed unused `PROD_LABELS`. |
| Target kill counted `chapters/*.md` | Switched to `meta/progress.json` `completed_chapters`. |
| Rewrites stat always 0 | Poll now parses `reviews/*.json` and counts `verdict == "rewrite"`. |
| Start button enabled for non-queued runs | Button now only active when `status === 'queued'`. |
| Race between target kill and manual stop | `killLocked` refuses to overwrite a `completed` status. |

## Verification

- `go build ./...` — pass
- `go vet ./...` — pass
- `go test ./internal/entry/web/...` — pass (67 tests)
- `node --check` on fixed JS files — pass
- `git diff --name-only` — only `internal/entry/web/` + existing fork-exception `server.go`/`run.go`

## Caveats

- Full `go test ./...` still fails on pre-existing unrelated tests in `internal/bootstrap`, `internal/notify`, and `internal/version`.
- MVP is intentionally limited: one run at a time, TXT only, no scheduling, no remote Continue for paused child processes.

## Status

Production Cockpit MVP implementation complete and reviewed.

## Post-review polish

- Moved **Hỗ trợ** tab to the end of the workspace tab bar; **Sản xuất** now sits between **Đánh giá** and **Hỗ trợ**.
- Added a vertical divider and heading underline to the Production left/right panes for clearer separation.
- Created user guide [`docs/production-cockpit.md`](docs/production-cockpit.md).
- Updated [`04-LUU-Y-MERGE-UPSTREAM.md`](04-LUU-Y-MERGE-UPSTREAM.md) with Production Cockpit post-merge notes.
- Fixed QA blockers:
  - Load `app-production.js` before `app-workspace.js` and guarded `loadProductionTab()` to prevent a boot crash when the last active tab was **Sản xuất**.
  - Enforced one running production job at a time in `prodRunRunner.start()`.
  - Ran `gofmt` on flagged files.
- Fixed non-blocker findings:
  - Added retry loop to `safeWriteFile` to tolerate transient Windows file locks on rename.
  - Fixed `scripts/model-spike-test.sh` binary path (`../../` → `../../../`) and made rewrite rate count real `verdict == "rewrite"` reviews.
  - Added `spike-test/` artifacts and `spike-reports/` to `.gitignore` and removed generated files.
  - Returned shallow copies from `prodRunStore` and surfaced persistence errors from `create`/`update`.
  - Replaced unbounded `ReadLogTail` with block-scanned `tailFileLines()`.
  - Preserved source permissions in `copyFile` and `Budget.WarnRatio` in `buildRunConfig`.
  - Capped `/api/prodruns/{id}/log?lines=` at 1000 and return 400 for invalid input.
  - Mapped export errors to correct HTTP status codes (404/400/500).
  - Made profile-path validation case-insensitive on Windows.
  - Added `profiles/` to `.gitignore` and documented the spike script as Unix-only.
  - Removed incorrect "budget exhausted" stop reason from the user guide.
  - Changed export chapter header from hardcoded Chinese "第 %s 章" to "Chapter %s".
  - Made `copyDirFiles` only ignore missing home-rules dir; permission/disk errors now surface.
  - Documented fragility of pause-marker detection in `hasPauseMarker`.
