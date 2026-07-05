# 04 — Lưu Ý Merge Upstream & Big Change History

**Fork:** ainovel-cli-fork  
**Upstream:** https://github.com/voocel/ainovel-cli  
**Mục đích:** Ghi lại quy trình merge an toàn và các thay đổi lớn từ upstream ảnh hưởng đến fork. Dùng để sau mỗi lần merge, dev biết behavior nào đổi và cần test lại.

---

## 1. Quy tắc Merge An Toàn Cho Fork

Fork này theo chiến lược **additive-only**. Chỉ được thêm file mới trong `internal/entry/web/`; không sửa `internal/host/`, `internal/tools/`, `assets/prompts/`, `internal/entry/tui/` trừ khi upstream bắt buộc (và phải ghi chú lý do).

Quy trình mỗi lần merge:

```bash
# 1. Lấy metadata mới nhất
git fetch upstream

# 2. Xem có commits mới không
git log --oneline HEAD..upstream/main

# 3. Dry-run merge để phát hiện conflict (không đụng working tree)
git merge-tree $(git merge-base HEAD upstream/main) HEAD upstream/main

# 4. Merge thật nếu dry-run sạch
git merge upstream/main --no-edit

# 5. Gate bắt buộc sau merge
go build ./...
go vet ./...
go test ./internal/entry/web/...
```

**Lưu ý đặc biệt:** Nếu upstream thay đổi `assets/prompts/`, phải **rebuild binary** — browser refresh không đủ vì prompts được embed qua `go:embed`.

---

## 2. v0.6.1 — Pause Points & Completion Convergence

**Upstream tag:** `v0.6.1`  
**Merge date:** 2026-07-03  
**Merge commit:** `4544b53`  
**Range:** `6342dc3..1a4b5c5`

### Big changes

| Feature | Files chính | Impact |
|---------|-------------|--------|
| **用户验收停靠点** (user acceptance pause point) | `internal/host/pause.go`, `internal/host/flow/pause.go`, `internal/tools/save_pause_point.go`, `internal/domain/runtime.go` | Sau rewrite, engine có thể tự động pause để user kiểm tra. Web UI cần đọc pause state từ snapshot để hiển thị nút Continue thay vì hiển thị idle. |
| **完本收敛** (book completion convergence) | `assets/prompts/architect-long.md`, `internal/tools/commit_chapter.go`, `internal/tools/save_volume_summary.go`, `internal/domain/story.go`, `docs/architecture.md` | Engine quyết định kết thúc sách thông minh hơn. Có khái niệm **收官卷** (final volume): khi story đã đến điểm kết, architect có thể append volume với `"final": true`; sau khi final volume viết xong và qua editor review, hệ thống tự động mark complete. |
| **Notify event mới** | `internal/notify/notify.go`, `config.example.jsonc` | Thêm `pause_point` vào danh sách notify events. |
| **TUI update** | `internal/entry/tui/*.go` | Hiển thị pause state. Không ảnh hưởng Web UI vì Web UI dùng snapshot riêng. |

### Lưu ý sau merge v0.6.1

- **Prompts đã đổi** → rebuild binary trước khi chạy bất kỳ novel nào.
- **Behavior hoàn thành sách khác** → spike test 30 chương phải chạy trên v0.6.1, không dùng kết quả cũ.
- **Pause point** có thể làm engine dừng giữa chừng sau rewrite. Khi làm Production Cockpit sau này, nên tận dụng behavior này thay vì tự xây stop logic phức tạp.
- **Final volume** có nghĩa là `TotalChapters` trong snapshot có thể thay đổi khi story tiến gần kết — automation target phải linh hoạt hoặc dựa trên `phase == complete` thay vì chapter count cứng.
- **Windows file lock với export/rename**: Trong spike test v0.6.1, engine export thất bại trên Windows khi IDE/file watcher giữ file chapter đang mở trong lúc `os.Rename` ghi đè. Lỗi này tồn thiởi trước v0.6.1, không phải regression. Cần đóng watcher/IDE khi chạy production, hoặc upstream cần thêm retry loop/copy-delete fallback cho Windows.

---

## 3. Production Cockpit MVP (tab Sản xuất)

**Ngày thêm:** 2026-07-03  
**Files mới toàn bộ trong `internal/entry/web/`:**
- `prodrun.go`, `prodrun_runner.go`, `prodrun_handlers.go`, `prodrun_profiles.go`, `prodrun_workspace.go`, `prodrun_export.go`
- `prodrun*_test.go`
- `assets/app-production.js`
- Cập nhật `assets/index.html`, `assets/app-workspace.js`, `assets/app-i18n.js`, `assets/app.css`, `embed.go`, `assets_test.go`
- Tiếp tục đụng fork-exception files: `server.go` (route registration), `run.go` (start wiring)

### Lưu ý sau merge khi có Production Cockpit

| Vùng | Tác động / Cần làm |
|------|--------------------|
| **Conflict** | Các file `prodrun*.go` và `app-production.js` nằm trong `internal/entry/web/`, không nằm trong upstream → merge upstream/main sẽ **không conflict** trên chúng. |
| **Rebuild binary** | Mọi thay đổi ở `assets/` đều cần `go build ./...` — browser refresh không đủ vì file được embed qua `go:embed`. |
| **Fork-exception files** | Nếu upstream sửa `internal/entry/web/server.go` hoặc `internal/entry/web/run.go`, phải reconcile lại việc đăng ký route `/api/prodruns*` và wiring `startWeb`/`web.NewServer`. Giữ additive-only: không xóa route cũ. |
| **`cmd/ainovel-cli/main.go`** | Upstream chưa có `--web` / `--headless`. Nếu upstream thêm CLI flags, đảm bảo `--web` vẫn start Web UI và `--headless` vẫn dùng được cho Production Cockpit runner. |
| **`meta/progress.json` schema** | Runner đọc field `completed_chapters` để biết khi nào đạt `targetChapters`. Nếu upstream đổi tên field hoặc format, cần cập nhật `readCompletedChapters`. |
| **`reviews/*.json` shape** | Runner đếm rewrites qua `verdict == "rewrite"`. Nếu upstream đổi cấu trúc review JSON, cần cập nhật `countReviewsAndRewrites`. |
| **Engine output path** | Cockpit dựa vào cwd-based output: mỗi run spawn `ainovel-cli --headless` với `Cmd.Dir = {runDir}`, nên output rơi vào `{runDir}/output/novel`. Nếu upstream thay đổi `bootstrap.Config.OutputDir` / `FillDefaults()` behavior, phải kiểm tra lại đường dẫn chapter/log/meta. |
| **Profile resolver** | Cockpit không còn chỉ đọc `./profiles/`. Nguồn chuẩn: `./.ainovel/profiles/` (`project/foo.md`) → `~/.ainovel/profiles/` (`global/foo.md`) → legacy `./profiles/` (`legacy/foo.md` / old `profiles/foo.md`). Sau merge phải giữ resolver chung trong `prodrun_profiles.go`; không quay lại `filepath.Join(repoRoot, profile)`. |
| **Resume mode** | `continue_workspace` seed workspace hiện tại sang sandbox rồi spawn `ainovel-cli --headless` **không `--prompt-file`** để engine native `Resume()` chạy tiếp. Không thêm logic resume vào `internal/host/` hay prompts. |
| **Target chapters** | Với `continue_workspace`, `targetChapters` là tổng số chương tuyệt đối cuối cùng, không phải delta. Sau merge phải test case đang có N chương và target > N. |
| **Fast-forward sync** | Continue sync mặc định là fast-forward: fingerprint host phải khớp seed fingerprint; diverge trả 409 và chỉ ghi khi user chọn `force`. Force phải backup `output/backups/pre-sync-*` trước khi ghi. |
| **Workspace noise exclude** | Fingerprint/seed/sync-back phải dùng cùng exclude list: bỏ `logs/`, `*.log`, `diag/`, `diagnostics/`, `exports/`, temp/lock files. Nếu upstream thêm log/diagnostic path mới, cập nhật `shouldExcludeWorkspaceSeed`. |
| **Pause point v0.6.1** | Cockpit chỉ đọc pause marker từ `run.log` (read-only). Nếu upstream cung cấp API pause tốt hơn (ví dụ snapshot pause state hoặc RPC), có thể thay thế polling log. |
| **Windows file lock** | Export TXT của Cockpit là server-side concat, không dùng `os.Rename`. Continue sync-back cũng không clear/rename cả thư mục host; dùng file-by-file `safeWriteFile` retry. Giữ behavior này, đừng chuyển sang `s.eng.Export()` hoặc `os.RemoveAll` full-replace vì sẽ re-introduce Windows lock/data-loss risk. |
| **Foundation Gate — detect phase** (2026-07-05) | `poll()` đọc `progress.json` field `phase`; khi `== "writing"` + `completed_chapters==0` + `fresh_profile` + chưa `FoundationApproved` → chuyển `awaiting_review` và kill child. Nếu upstream **đổi tên/giá trị field `phase`** (hiện `"writing"`, hằng `domain.PhaseWriting`) thì phải cập nhật `readWorkspacePhase`. Best-effort (poll 5s) — không phải hard gate; xem journal `260705`. |
| **Status `awaiting_review`** | Là schema mới của `ProdRun`. `load()` **cố ý KHÔNG** coalesce nó → `failed` như running/paused (child đã chủ động kill, không có process treo). Nếu refactor `load()`, giữ awaiting_review sống sót qua restart. |
| **Endpoint Gate mới** (fork-exception `server.go`) | `POST /api/prodruns/{id}/{approve,reject,revise,reveal}` + `GET .../foundation`. Approve = restart cùng run dir (native Resume, skip `--prompt-file` nhờ `runDirHasExistingOutput`). Revise = ghép `RevisionNote` vào cuối `profile.md` rồi tạo run mới. Sau merge nếu reconcile `server.go`, giữ đủ 5 route này. |
| **Reveal loopback-only** | `POST .../reveal` mở `runDir/output/novel` bằng `revealOpen`; tự chặn 403 khi bind non-loopback (giống `handleReveal`). Đừng nới cho public bind. |
| **Profile Library** (2026-07-05, `profiles_library.go`) | `GET /api/profiles/content`, `POST /api/profiles/{save,delete}`. Save **project-only** + `409` chống ghi đè âm thầm; delete **project-only** (guard `isWithinDir(resolved, projectBase)` → 403 cho global/legacy). Tái dùng `resolveExistingProfilePath`/`sanitizeFileName`. Nếu upstream đổi layout `profiles/` hay resolver, kiểm lại 3 endpoint này. |
| **Profile Studio C-lite** (`profile_studio.go`) | `POST /api/profiles/generate` tự dựng `bootstrap.NewModelSet(cfg)` (public constructor — KHÔNG đụng host) rồi `ForRole("thinking").GenerateStream`. Nếu upstream đổi chữ ký `NewModelSet` / `ModelSet.ForRole` / agentcore stream API, vá `runProfileGeneration`. System prompt principle-based, cố ý không ví dụ cụ thể. |

### Production profile path contract

API `/api/profiles` trả:

```json
{ "name": "foo.md", "path": "project/foo.md", "source": "project" }
```

`ProdRun.Profile` lưu nguyên `path` này. Runner resolve lúc start, không resolve/copy tại create. Contract cần giữ:

| Ref | Resolve tới | Ghi chú |
|---|---|---|
| `project/foo.md` | `./.ainovel/profiles/foo.md` | profile riêng project hiện tại |
| `global/foo.md` | `~/.ainovel/profiles/foo.md` | profile dùng lại giữa project |
| `legacy/foo.md` | `./profiles/foo.md` | legacy/sample |
| `profiles/foo.md` | `./profiles/foo.md` | backward compat cho job cũ |

Security guard bắt buộc: reject absolute path, unknown source, non-`.md`, traversal, và symlink escape khỏi profile root.

---

## 4. Checklist Sau Mỗi Lần Merge

- [ ] `git fetch upstream` xong.
- [ ] Dry-run merge không báo conflict.
- [ ] `git merge upstream/main` thành công.
- [ ] `go build ./...` pass.
- [ ] `go vet ./...` pass.
- [ ] `go test ./internal/entry/web/...` pass.
- [ ] Đọc diff của `docs/architecture.md` nếu upstream thêm section mới.
- [ ] Kiểm tra `assets/prompts/` có thay đổi gì không.
- [ ] Rebuild binary nếu prompts/assets thay đổi.
- [ ] Chạy smoke test 1 chapter nếu prompts thay đổi nhiều.
- [ ] Với Production Cockpit fresh: kiểm tra `/api/profiles` vẫn list `project/global/legacy`, tạo/start `fresh_profile` từ profile.
- [ ] Với Production Cockpit resume: tạo `continue_workspace` từ workspace có progress, start không `--prompt-file`, sync fast-forward được, `logs/*.log` không làm fingerprint diverge.
- [ ] Với force sync: kiểm tra có backup `output/backups/pre-sync-*` trước khi ghi đè.
- [ ] Với Foundation Gate: tạo `fresh_profile`, start, chờ tới `awaiting_review`; kiểm tra `GET /api/prodruns/{id}/foundation` (+`?section=world|characters`) trả đúng; Approve resume được (không re-gate); Reject xoá; Revise tạo run mới có note trong `profile.md`; Reveal mở đúng `output/novel`.
- [ ] Với Profile Library: save profile mới (project), sửa + save có 409/confirm ghi đè, delete project OK nhưng delete `global/legacy` trả 403; Studio `generate` trả markdown vào editor (không tự lưu/chạy).

---

## 5. Lịch Sử Thay Đổi Lớn

| Version | Date | Key Change | Action Required |
|---------|------|------------|-----------------|
| v0.6.1 | 2026-07-03 | Pause points + completion convergence | Rebuild binary; rerun spike test; update Web UI pause handling |
| post-v0.6.1 (fork) | 2026-07-03 | Production Cockpit MVP (tab Sản xuất) | Rebuild binary; check `server.go`/`run.go` after upstream merge; verify `progress.json`/`reviews/*.json` schema |
| post-v0.6.1 (fork) | 2026-07-04 | Production Cockpit resume mode (`continue_workspace`) + fast-forward sync | Verify native headless `Resume()`, seed fingerprint, `logs/` exclude, force backup, and Windows-safe file-by-file sync |
| post-v0.6.1 (fork) | 2026-07-04 | Production profile resolver standardized to `.ainovel` 2-layer model + legacy fallback | Verify `/api/profiles`, profile path validation, and `prepareRunDir` resolver after upstream merge |
| post-v0.6.1 (fork) | 2026-07-05 | Foundation Gate Milestone 1a (`awaiting_review` + approve/reject/revise/reveal, best-effort poll on `progress.json` phase) | Verify `readWorkspacePhase`, `FoundationApproved` chống re-gate, 5 endpoint Gate trong `server.go`, reveal loopback-only |
| post-v0.6.1 (fork) | 2026-07-05 | Profile Library + Profile Studio C-lite (author/generate profiles in UI) | Verify 4 endpoint `/api/profiles/{content,save,delete,generate}` trong `server.go`; save/delete project-only; `NewModelSet(cfg)` cho Studio |
---

## 6. Link

- Report merge test chi tiết: [`plans/reports/upstream-merge-test-260703-1121-v061-pause-convergence-report.md`](plans/reports/upstream-merge-test-260703-1121-v061-pause-convergence-report.md)
- Architecture upstream: [`docs/architecture.md`](docs/architecture.md)
- Journal Production Cockpit MVP (kèm sơ đồ tương tác + state machine): [`docs/journals/260703-production-cockpit-mvp.md`](docs/journals/260703-production-cockpit-mvp.md)
- Journal Foundation Gate (best-effort gate, revise/reveal, race, bug đã fix): [`docs/journals/260705-foundation-gate.md`](docs/journals/260705-foundation-gate.md)
- Journal Profile Library & Studio (tạo/sinh/lưu profile trong UI): [`docs/journals/260705-profile-library-studio.md`](docs/journals/260705-profile-library-studio.md)
- Hướng dẫn dùng Production Cockpit: [`docs/production-cockpit.md`](docs/production-cockpit.md)
